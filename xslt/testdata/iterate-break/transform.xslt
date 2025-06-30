<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:iterate select="/root/item/value">
			<xsl:on-completion>
				<total><xsl:value-of select="$sum"/></total>
			</xsl:on-completion>
			<xsl:param name="sum" select="0"/>
			<xsl:if test="$sum >= 35">
				<xsl:break>
					<total>
						<xsl:value-of select="$sum div 5"/>
					</total>
				</xsl:break>
			</xsl:if>
			<xsl:next-iteration>
				<xsl:with-param name="sum" select="$sum + number(.)"/>
			</xsl:next-iteration>
		</xsl:iterate>
	</xsl:template>
</xsl:stylesheet>